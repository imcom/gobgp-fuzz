/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	. "github.com/imcom/gobgp-fuzz/internal/pkg"
	api "github.com/osrg/gobgp/v3/api"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	Addr       string
	Concurrent int
	ctx        context.Context
	cancel     context.CancelFunc
	Cidrs      []string
)

// remoteCmd represents the remote command
var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			// no duration means run forever
			return nil
		}

		_, err := strconv.Atoi(args[0])
		if err != nil {
			logger.Sugar().Errorf("duration should be an integer but `%v` given", args[0])
			return errors.New("type error, integer required")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		var duration int
		if len(args) >= 1 {
			duration, _ = strconv.Atoi(args[0])
		}

		ctx, cancel = context.WithCancel(context.Background())
		conn, err := grpc.DialContext(ctx, Addr, grpc.WithInsecure())
		if err != nil {
			logger.Sugar().Infof("fail to connect to gobgp with error: %+v", err)
			os.Exit(1)
		}
		client := api.NewGobgpApiClient(conn)
		if _, err := client.GetBgp(context.TODO(), &api.GetBgpRequest{}); err != nil {
			logger.Sugar().Infof("fail to get gobgp info with error: %+v", err)
			os.Exit(1)
		}
		logger.Sugar().Info("connected to gobgpd for testing @", Addr)

		wg := sync.WaitGroup{}

		// init workers
		workers := []Worker{}

		workerCtx, workerCancel := context.WithCancel(ctx)
		dumper := NewDumper(workerCtx, client, Concurrent, logger.Sugar(), "rib-dumper", &wg)
		workers = append(workers, dumper)

		advertiser := NewAdvertiser(workerCtx, client, Concurrent, logger.Sugar(), "rib-modifier", Cidrs, &wg)
		workers = append(workers, advertiser)

		// start workers
		for _, w := range workers {
			w.Loop()
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

		go func() {
			defer workerCancel()
			if duration > 0 {
				logger.Sugar().Infof("fuzz will run for %d seconds", duration)
				timer := time.NewTimer(time.Duration(duration) * time.Second)
				select {
				case <-timer.C:
					break
				case <-sigs:
					break
				}
			} else {
				logger.Sugar().Infof("fuzz has begun, hit Ctrl-C to stop")
				<-sigs
			}
		}()

		// wait for workers
		wg.Wait()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if cancel != nil {
			logger.Sugar().Info("fuzz finished, clean up the scene")
			cancel()
		}
	},
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	rootCmd.AddCommand(remoteCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// remoteCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	remoteCmd.Flags().StringVarP(&Addr, "addr", "a", "127.0.0.1:50051", "gobgpd grpc addr")
	remoteCmd.Flags().IntVarP(&Concurrent, "concurrent", "c", runtime.NumCPU(), "concurrent callers")
	remoteCmd.Flags().StringSliceVarP(&Cidrs, "cidrs", "s", []string{"10.0.0.0/8"}, "CIDR pool to generate prefixes")
}
