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

var (
	allowedIPs  []string
	wireGuardIP string
)

// joinCmd represents the join command
var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "join this client as a peer to the k8s wireguard server",
	Long: `'join' will setup a local WireGuard link device, wg0, and configure it
	to join the server as a peer. It will also create routes for any additional IPs
	passed in the allowed-ips. For direct access to k8s resources, pass the pod and service
	CIDRs of the k8s cluster to allowed-ips so that traffic to those IPs will be routed appropriately.`,
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
		p := deployer.NewClient(client, allowedIPs, wireGuardIP)
		p.JoinServerAsPeer(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
	joinCmd.Flags().StringSliceVar(&allowedIPs, "allowed-ips", []string{}, "additional IP ranges to send through the tunnel. ex: 192.168.0.0/16,10.244.1.0/24")
	joinCmd.Flags().StringVar(&wireGuardIP, "wireguard-ip", "100.120.220.2/24", "IP to be used for the WireGuard space for this peer.")
}
