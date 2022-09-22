# Install agent

To install idemeum agent:

```bash
sudo ./install
```

## To run the agent as systemd service
To run the idemeum agent with the downloaded yaml config file:

```bash 
sudo ./agent-setup <agentName>
```

Copy the downloaded agent config to /etc/<agentName>.yaml


## To run the agent using command
To run the idemeum agent with the downloaded yaml config file:

```bash
idemeum start --config= <yaml_config_file_path>
```

# More details
* [Idemeum documentation](https://docs.idemeum.com/remote-access/secure-remote-access-overview.html)


