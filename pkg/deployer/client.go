package deployer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type WireGuard struct {
	netlink.LinkAttrs
}

func (w *WireGuard) Attrs() *netlink.LinkAttrs {
	return &w.LinkAttrs
}

func (w *WireGuard) Type() string {
	return "wireguard"
}

type client struct {
	k8s        *kubernetes.Clientset
	allowedIPs []string
	peerIP     string
}

func NewClient(cs *kubernetes.Clientset, allowedIPs []string, peerIP string) *client {
	return &client{
		k8s:        cs,
		allowedIPs: allowedIPs,
		peerIP:     peerIP,
	}
}

func (c *client) JoinServerAsPeer(ctx context.Context) {
	// check if this client already has a wg0 interface
	c.ensureLocalWireguardLink(ctx)

	// configure device to be a peer of the k8s wg server
	dev := c.ensureLinkDeviceConfigured(ctx)

	// add peer to server config
	log.Default().Printf("got device %s with public key %s", dev.Name, dev.PublicKey)
	c.ensurePeerOnServer(ctx, dev)
}

func (c *client) ensurePeerOnServer(ctx context.Context, wgDev *wgtypes.Device) {
	wgSec, err := c.k8s.CoreV1().Secrets(kubeSystemNS).Get(ctx, wireguardSecretName, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	base64Sec := wgSec.Data[serverConfigTemplateFile]
	if len(base64Sec) == 0 {
		log.Fatalf("expected data, got nothing")
	}

	if strings.Contains(string(base64Sec), wgDev.PublicKey.String()) {
		log.Default().Printf("server already configured with peer %s, skipping rolling server pods", wgDev.PublicKey)
		return
	}

	updatedSec := strings.Builder{}
	updatedSec.WriteString(string(base64Sec))
	updatedSec.WriteString("\n")
	updatedSec.WriteString("[Peer]\n")
	updatedSec.WriteString(fmt.Sprintf("PublicKey = %s\n", wgDev.PublicKey))
	updatedSec.WriteString(fmt.Sprintf("AllowedIPs = %s\n", c.peerIP))

	// update the secret
	wgSec.Data[serverConfigTemplateFile] = []byte(updatedSec.String())

	_, err = c.k8s.CoreV1().Secrets(kubeSystemNS).Update(ctx, wgSec, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("failed to update secret %s", err)
	}

	deployment, err := c.k8s.AppsV1().Deployments(kubeSystemNS).Get(ctx, wireguardSecretName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("failed to get deployment %s", err)
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = map[string]string{}
	}

	deployment.Spec.Template.Annotations["restart.at/time"] = time.Now().String()

	_, err = c.k8s.AppsV1().Deployments(kubeSystemNS).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("failed to update deployment %s", err)
	}
}

func (c *client) ensureLinkDeviceConfigured(ctx context.Context) *wgtypes.Device {
	wgSvc, err := c.k8s.CoreV1().Services("kube-system").Get(ctx, "wireguard", metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	if len(wgSvc.Status.LoadBalancer.Ingress) == 0 {
		log.Fatal("no external IPs found on wireguard service, cant add as a peer")
	}

	wgSec, err := c.k8s.CoreV1().Secrets("kube-system").Get(ctx, "wireguard", metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	if wgSec.Annotations["azure.kubernetes.com/publickey"] == "" {
		log.Fatalf("public key annotation missing from secret %s/%s", wgSec.Namespace, wgSec.Name)
	}

	serverPubKey, err := wgtypes.ParseKey(wgSec.Annotations["azure.kubernetes.com/publickey"])
	if err != nil {
		log.Fatal(err)
	}

	wgClient, err := wgctrl.New()
	if err != nil {
		log.Fatal(err)
	}

	dev, err := wgClient.Device("wg0")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("failed to check device wg0: %s", err)
	}

	serverAllowedIPs := []net.IPNet{}

	// peer IP
	_, ipNet, err := net.ParseCIDR(c.peerIP)
	if err != nil {
		log.Fatalf("failed to parse peerIP as CIDR: %s", err)
	}
	serverAllowedIPs = append(serverAllowedIPs, *ipNet)

	// any allowed-ips from the cli
	for _, ip := range c.allowedIPs {
		_, ipNet, err = net.ParseCIDR(ip)
		if err != nil {
			log.Fatalf("failed to parse peerIP as CIDR: %s", err)
		}

		serverAllowedIPs = append(serverAllowedIPs, *ipNet)
	}

	var cfg wgtypes.Config
	if dev == nil || dev.PrivateKey.String() == "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		log.Default().Println("device wg0 not initialized, setting up for first time")
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			log.Fatalf("could not generate new private key for wg0: %s", err)
		}

		cfg = wgtypes.Config{
			PrivateKey: &key,
			Peers: []wgtypes.PeerConfig{
				{
					PublicKey: serverPubKey,
					Endpoint: &net.UDPAddr{
						IP:   net.ParseIP(wgSvc.Status.LoadBalancer.Ingress[0].IP),
						Port: int(wgSvc.Spec.Ports[0].Port),
					},
					AllowedIPs: serverAllowedIPs,
				},
			},
		}
		err = wgClient.ConfigureDevice("wg0", cfg)
		if err != nil {
			log.Fatal(err)
		}

		dev, err = wgClient.Device("wg0")
		if err != nil {
			log.Fatal(err)
		}
	}

	serverFound := false
	for _, p := range dev.Peers {
		if p.PublicKey == serverPubKey {
			serverFound = true
		}
	}

	if !serverFound {
		log.Default().Printf("server key %s not found as peer, adding to wg0", serverPubKey)

		cfg = wgtypes.Config{
			PrivateKey: &dev.PrivateKey,
			Peers:      []wgtypes.PeerConfig{},
		}

		for _, p := range dev.Peers {
			cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
				PublicKey:  p.PublicKey,
				AllowedIPs: p.AllowedIPs,
				Endpoint:   p.Endpoint,
			})
		}

		// add server
		cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
			PublicKey: serverPubKey,
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(wgSvc.Status.LoadBalancer.Ingress[0].IP),
				Port: int(wgSvc.Spec.Ports[0].Port),
			},
			AllowedIPs: serverAllowedIPs,
		})
		err = wgClient.ConfigureDevice("wg0", cfg)
		if err != nil {
			log.Fatal(err)
		}

		dev, err = wgClient.Device("wg0")
		if err != nil {
			log.Fatal(err)
		}
	}

	return dev
}

func (c *client) ensureLocalWireguardLink(ctx context.Context) {
	la := netlink.NewLinkAttrs()
	la.Name = "wg0"
	dev := &WireGuard{LinkAttrs: la}
	err := netlink.LinkAdd(dev)
	if err != nil && !errors.Is(err, os.ErrExist) {
		panic(err)
	}

	link, err := netlink.LinkByName("wg0")
	if err != nil {
		log.Fatal(err)
	}

	// add peerIP as address
	_, ipNet, err := net.ParseCIDR(c.peerIP)
	if err != nil {
		log.Fatalf("could not parse peerIP %s as CIDR: %s", c.peerIP, err)
	}

	addr := netlink.Addr{
		IPNet: ipNet,
	}
	err = netlink.AddrAdd(link, &addr)
	if err != nil && !errors.Is(err, os.ErrExist) {
		log.Fatal(err)
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		log.Fatalf("failed to bring up link: %s", err)
	}

	// add routes for additional allowed-ips
	for _, ip := range c.allowedIPs {
		_, ipNet, err := net.ParseCIDR(ip)
		if err != nil {
			log.Fatalf("failed to parse allowed-ip %s as CIDR: %s", ip, err)
		}

		rt := netlink.Route{
			Dst:       ipNet,
			LinkIndex: link.Attrs().Index,
		}

		err = netlink.RouteAdd(&rt)
		if err != nil && !errors.Is(err, os.ErrExist) {
			log.Fatalf("failed to add route %s", err)
		}
	}
}
