/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)



// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kwg",
	Short: "Kubernetes WireGuard toolkit.",
	Long: `kwg is a cli tool for quickly deploying a WireGuard server to a Kubernetes cluster 
	and configuring peers on the server. The wg server will act as a subnet router to the rest
	of the cluster so peers can access pods and services directly without exposing a public endpoint.
	The only public piece is the WireGuard LoadBalancer service which allows UDP traffic on port 51820
	but any traffic not coming from a valid peer will be dropped.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig for the cluster")
}


