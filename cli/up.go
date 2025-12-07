package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/DataDrake/cli-ng/v2/cmd"
	hsnode "github.com/hyprspace/hyprspace/node"
	"github.com/ipfs/go-log/v2"
)


var logger = log.Logger("hyprspace")

// Up creates and brings up a Hyprspace Interface.
var Up = cmd.Sub{
	Name:  "up",
	Alias: "up",
	Short: "Create and Bring Up a Hyprspace Interface.",
	Run:   UpRun,
}

// UpRun handles the execution of the up command.
func UpRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}

	// Parse Global Config Flag for Custom Config Path
	configPath := r.Flags.(*GlobalFlags).Config
	if configPath == "" {
		configPath = "/etc/hyprspace/" + ifName + ".json"
	}

	log.SetLogLevel("hyprspace", "info")
	log.SetLogLevelRegex("^hyprspace/", "info")

	node := hsnode.New(context.Background(), configPath, ifName)
	checkErr(node.Run())
	logger.Info("Node ready")

	exitCh := make(chan os.Signal, 1)
	rebootstrapCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	signal.Notify(rebootstrapCh, syscall.SIGUSR1)

	for {
		select {
		case <-rebootstrapCh:
			logger.Info("Rebootstrapping on SIGUSR1")
			node.Rebootstrap()
		case <-exitCh:
			logger.Info("Shutting down...")
			go func() {
				<-exitCh
				logger.Fatal("Terminating immediately")
			}()
			checkErr(node.Stop())
			os.Exit(0)
		}
	}
}
