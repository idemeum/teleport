# Systemd Service

Sample configuration of `systemd` service file for idemeum

To start idemeum agent as daemon:

```bash
sudo cp idemeum.service /etc/systemd/system/idemeum.service
sudo systemctl daemon-reload
sudo systemctl enable idemeum
sudo systemctl start idemeum
```

To check on idemeum daemon status:

```bash
systemctl status idemeum
```

To take a look at idemeum system log:

```bash
journalctl -fu idemeum
```

