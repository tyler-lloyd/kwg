/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/tyler-lloyd/kwg/pkg/deployer"
)



// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys a wireguard server to the k8s cluster.",
	Long: `Deploys the WireGuard server along with exposing a LoadBalancer service
	so peers are able to connect externally.`,
	Run: func(cmd *cobra.Command, args []string) {
		var cfg *rest.Config
		if kubeconfig == "" {
			cfg = config.GetConfigOrDie()
		} else {
			var err error
			cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				panic(err)
			}
		}

		client := kubernetes.NewForConfigOrDie(cfg)
		deployer := deployer.New(client)
		deployer.DeployServer(cmd.Context())
	},
}

var kubeconfig string

func init() {
	rootCmd.AddCommand(deployCmd)
}
