package deployer

var serverConfigTemplate string = `[Interface]
Address = 100.120.220.1/24
ListenPort = 51820
PrivateKey = {PRIVATE_KEY}
PostUp = iptables -A FORWARD -i ENI -j ACCEPT; iptables -A FORWARD -o ENI -j ACCEPT; iptables -t nat -A POSTROUTING -o ENI -j MASQUERADE
PostUp = sysctl -w -q net.ipv4.ip_forward=1
PostDown = iptables -D FORWARD -i ENI -j ACCEPT; iptables -D FORWARD -o ENI -j ACCEPT; iptables -t nat -D POSTROUTING -o ENI -j MASQUERADE
PostDown = sysctl -w -q net.ipv4.ip_forward=0
`

var serverDeployment string = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: wireguard
  namespace: wireguard
spec:
  selector:
    matchLabels:
      name: wireguard
  template:
    metadata:
      labels:
        name: wireguard
    spec:
      initContainers:
        # The exact name of the network interface needs to be stored in the
        # wg0.conf WireGuard configuration file, so that the routes can be
        # created correctly.
        # The template file only contains the "ENI" placeholder, so when
        # bootstrapping the application we'll need to replace the placeholder
        # and create the actual wg0.conf configuration file.
        - name: "wireguard-template-replacement"
          image: "busybox"
          command: ["sh", "-c", "ENI=$(ip route get 8.8.8.8 | grep 8.8.8.8 | awk '{print $5}'); sed \"s/ENI/$ENI/g\" /etc/wireguard-secret/wg0.conf.template > /etc/wireguard/wg0.conf; chmod 400 /etc/wireguard/wg0.conf"]
          volumeMounts:
            - name: wireguard-config
              mountPath: /etc/wireguard/
            - name: wireguard-secret
              mountPath: /etc/wireguard-secret/

      containers:
        - name: "wireguard"
          image: "linuxserver/wireguard:latest"
          ports:
            - containerPort: 51820
          env:
            - name: "TZ"
              value: "UTC"
            # Keep the PEERS environment variable to force server mode
            - name: "PEERS"
              value: "1"
          volumeMounts:
            - name: wireguard-config
              mountPath: /etc/wireguard/
              readOnly: true
          securityContext:
            privileged: true
            capabilities:
              add:
                - NET_ADMIN

      volumes:
        - name: wireguard-config
          emptyDir: {}
        - name: wireguard-secret
          secret:
            secretName: wireguard

      imagePullSecrets:
        - name: docker-registry
`