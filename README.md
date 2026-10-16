# Meteor
> Meteor is an access rule management software based on GeoIP, simpler and easier to use compared to Iptables, and supports both UDP and TCP protocols.
## Usage

**Installation**
```shell
meteor install
```

**Modify Configuration File**

```shell
vim /etc/meteor/meteor.yaml
```

Configuration file example:
```shell
location:
  type: geoip                 # 目前仅支持 geoip
  file: GeoLite2-City.mmdb    # 配置geoip后支持按城市配置规则, 数据库文件需自行下载，然后配置文件地址
forwarders:
  - protocol: tcp             # 仅支持 tcp 和 udp
    addr: ":54321"            # 本机监听地址
    to: 127.0.0.1:12345       # 目标地址
    rules:
      - city: beijing,成都     # 城市，支持中文、拼音
        allowed: true         # 是否允许访问 ✅
      - ip: 0.0.0.0/0         # 0.0.0.0/0 代表全部的IP地址
        allowed: false        # 这个配置的含义就是只允许 beijing和成都的IP地址访问，其他的全部禁止访问。🈲
  - protocol: udp
    addr: ":54321"
    to: 127.0.0.1:12345
proxies:
  - protocol: http           # 仅支持 http、https、socks5
    addr: 127.0.0.1:80       # 本地监听地址
    auth: true               # 是否开启认证
    accounts:                # 账户列表
      - username: a          # 账号
        password: b          # 密码
  - protocol: https
    addr: 127.0.0.1:443   
    key: /root/key.pem       # https key path
    cert: /root/cert.pem     # https cert path
    auth: true               # 是否开启认证
    accounts:                # 账户列表
      - username: a          # 账号
        password: b          # 密码
  - protocol: socks5
    addr: 127.0.0.1:1080
    auth: true               # 是否开启认证
    accounts:                # 账户列表
      - username: a          # 账号
        password: b          # 密码
```

start
```shell
meteor start
```

stop
```shell
meteor stop
```

uninstall
```shell
meteor uninstall
```

## Other Parameters

```shell 
meteor -h
```

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
Dynamic Debug Mode
```shell
meteor install -d
```
View Running Logs
```shell
journalctl -u meteor -f
```

Logs use the Go standard library `log/slog` package with a structured text handler writing to standard error. The application does not create or rotate log files. Enable debug logs with `-d`. On Linux, systemd collects stdout and stderr in the journal by default: use `journalctl -u meteor -f` to follow logs or `journalctl -u meteor -b` to inspect logs from the current boot. Journald manages retention and rotation. Other service managers need stderr capture configured; Windows Event Log does not automatically collect stderr.

## TODO List
- Status statistics function
  - Connection times, total, total data transmission volume, top-level IP access, top-level IP rejection, and other information statistics and display (command line)
- Structured logging
  - Structured recording of connection logs for unified log takeover
  - Log Elasticsearch bridging
- Threat intelligence bridging
  - download
  - Upload

## Development

Requires Go 1.27.2 or later.

```shell
go mod download
go test -race ./...
go vet ./...
go build ./...
```

Configuration is a single, strictly validated YAML document. Unknown fields, invalid IP rules, unsupported protocols, missing HTTPS certificates, and authentication without accounts prevent startup. City rules require `location.type: geoip` and a readable `location.file`; GeoIP initialization errors prevent startup. IP and city lists accept comma-separated values with surrounding whitespace. Rules are evaluated in order; the first matching rule wins, and unmatched clients are allowed.

Generated configuration listens on `127.0.0.1:8080` and forwards to `127.0.0.1:80`. Edit the listen address explicitly to expose it externally. Config files are created with mode `0600` and existing files are preserved. SOCKS5 supports TCP CONNECT, with optional username/password authentication.
