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
	"strconv"
	"time"

	. "github.com/imcom/gobgp-fuzz/internal/pkg"
	api "github.com/osrg/gobgp/v3/api"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	Addr    string
	Debug   bool
	Workers int
	ctx     context.Context
	cancel  context.CancelFunc
)

var logger, _ = zap.NewProduction()

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
			return errors.New("requires a color argument")
		}

		_, err := strconv.Atoi(args[0])
		if err != nil {
			logger.Sugar().Errorf("duration should be an integer but `%v` given", args[0])
			os.Exit(1)
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		duration, _ := strconv.Atoi(args[0])

		logger.Sugar().Infof("connecting to gobgpd @ %s for testing\n", Addr)
		ctx, cancel = context.WithCancel(context.Background())
		conn, err := grpc.DialContext(ctx, Addr, grpc.WithInsecure())
		if err != nil {
			logger.Sugar().Infof("fail to connect to gobgp with error: %+v\n", err)
			os.Exit(1)
		}
		client := api.NewGobgpApiClient(conn)
		if _, err := client.GetBgp(context.TODO(), &api.GetBgpRequest{}); err != nil {
			logger.Sugar().Infof("fail to get gobgp info with error: %+v\n", err)
			os.Exit(1)
		}

		dumperCh := make(chan struct{}, 1)
		dumperCtx, dumperCancel := context.WithCancel(ctx)
		dumper := NewDumper(dumperCtx, client, Workers, dumperCh, logger.Sugar())
		// start dumper
		go dumper.Run()

		timer := time.NewTimer(time.Duration(duration) * time.Second)
		<-timer.C
		dumperCancel()
		// wait for dumper
		<-dumperCh
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// flush all logs, if any
		logger.Sync()
		if cancel != nil {
			logger.Sugar().Info("fuzz finished, clean up the scene")
			cancel()
		}
	},
}

func init() {
	rootCmd.AddCommand(remoteCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// remoteCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	remoteCmd.Flags().BoolVarP(&Debug, "debug", "d", false, "use debug")
	remoteCmd.Flags().StringVarP(&Addr, "addr", "a", "127.0.0.1:50051", "gobgpd grpc addr")
	remoteCmd.Flags().IntVarP(&Workers, "workers", "w", 2, "concurrent caller workers")
}
