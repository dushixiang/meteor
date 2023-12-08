# Meteor
[English](README.md)  ｜  [中文](README_zh.md)
## Introduction
Meteor is a transport layer proxy tool with basic functionalities such as port forwarding, HTTP/SOCKS5 proxy, etc.
Beyond these basic features, Meteor focuses more on usability and network security, with optimizations and improvements. 
For example, it offers persistent process services, and access rules can be set based on GeoIP for enhanced security. 
Future updates will include additional security features such as threat intelligence integration, rate limiting, unified log integration, etc.

Our Goal:
To create a simple-to-use, default-secure network proxy tool.

## Usage

**Installation**
```shell
meteor install
```

**Modify the Configuration File**

```shell
vim /etc/meteor/meteor.yaml
```

Configuration file example：
```shell
location:
  type: geoip                 # Currently only supports GeoIP
  file: GeoLite2-City.mmdb    # After configuring GeoIP, rules can be set based on cities. The database file needs to be downloaded and the file path specified in the configuration.
forwarders:
  - protocol: tcp             # Only supports tcp and udp
    addr: ":54321"            # Local listening address
    to: 127.0.0.1:12345       # Target address
    rules:
      - city: beijing,chengdu  # City, supports Chinese and Pinyin
        allowed: true         # Allow access ✅
      - ip: 0.0.0.0/0         # 0.0.0.0/0 represents all IP addresses
        allowed: false        # This configuration means only allow access from IP addresses in Beijing and Chengdu, all others are denied. 🈲
  - protocol: udp
    addr: ":54321"
    to: 127.0.0.1:12345
proxies:
  - protocol: http           # Only supports http, https, socks5
    addr: 127.0.0.1:80       # Local listening address
    auth: true               # Enable authentication
    accounts:                # Account list
      - username: a          # Username
        password: b          # Password
  - protocol: https
    addr: 127.0.0.1:443   
    key: /root/key.pem       # HTTPS key path
    cert: /root/cert.pem     # HTTPS cert path
    auth: true               # Enable authentication
    accounts:                # Account list
      - username: a          # Username
        password: b          # Password
  - protocol: socks5
    addr: 127.0.0.1:1080
    auth: true               # Enable authentication
    accounts:                # Account list
      - username: a          # Username
        password: b          # Password
```

Start
```shell
meteor start
```

Stop
```shell
meteor stop
```

Uninstall
```shell
meteor uninstall
```

## Other Parameters

`meteor -h`

```shell
Meteor is a network tool that can quickly forward tcp and udp ports and start http, https and socks5 proxy servers.

Usage:
  meteor [flags]
  meteor [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  install     Install meteor as a system service
  restart     Restart meteor system service
  start       Start meteor system service
  stop        Stop meteor system service
  uninstall   Uninstall meteor system service
  version     Show version

Flags:
  -c, --config string   -c /path/config.yaml (default "/etc/meteor/meteor.yaml")
  -d, --debug           print debug log
  -h, --help            help for meteor

Use "meteor [command] --help" for more information about a command.
```

### Example
Enable Debug mode
```shell
meteor install -d
```
View running logs
```shell
journalctl -u meteor -f
```

## TODO List
- Status statistics
  - Connection counts, total counts, total data transmitted, top-level IP access, top-level IP rejections, etc. statistics and display (command line)
- Structured logs
  - Structured recording of connection logs for unified log management
- Threat intelligence integration
  - Integration
  - Threat Intelligence Reporting
- QoS/Rate limiting
  - Concurrent limitations
  - Total bandwidth limitations
- Weak network simulation
  - Packet loss simulation
  - Traffic bandwidth limitation based on IP address
