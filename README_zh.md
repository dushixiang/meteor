# Meteor
> Meteor是一款基于GeoIP的访问规则管理软件, 相比于Iptables更加简单易用。
## 使用

**安装**
```shell
meteor install
```

**修改配置文件**

```shell
vim /etc/meteor/meteor.yaml
```

配置文件示例：
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

启动
```shell
meteor start
```

停止
```shell
meteor stop
```

卸载
```shell
meteor uninstall
```

## 其他参数

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
动态Debug模式
```shell
meteor install -d
```
查看运行日志
```shell
journalctl -u meteor -f
```

日志通过 Go 标准库 `log/slog` 以结构化文本格式输出到标准错误，程序不创建或轮转日志文件。使用 `-d` 开启调试日志。Linux systemd 默认将标准输出和标准错误收集到 journal，可用 `journalctl -u meteor -f` 实时查看，或用 `journalctl -u meteor -b` 查看本次启动后的日志；日志保留和轮转由 journald 管理。其他服务管理器需要配置标准错误收集，Windows Event Log 不会自动收集标准错误。

## TODO List
- 状态统计功能
  - 连接次数，总数，总共传输数据量， 访问top级IP，拒绝top级IP 等信息统计及展示(命令行)
- 结构化日志
  - 连接日志结构化记录，方便统一日志接管
  - 日志Elasticsearch桥接
- 威胁情报接入
  - 接入
  - 上传

## 开发与配置校验

需要 Go 1.27.2 或更高版本。

```shell
go mod download
go test -race ./...
go vet ./...
go build ./...
```

配置文件必须是单个 YAML 文档，并进行严格校验。未知字段、非法 IP 规则、不支持的协议、HTTPS 缺少证书或私钥、开启认证却没有账户，都会导致启动失败。城市规则必须配置 `location.type: geoip` 和可读取的 `location.file`；GeoIP 数据库初始化失败会终止启动。IP 和城市列表支持用逗号分隔，忽略各项两侧空格。规则按顺序匹配，首个匹配规则生效，没有匹配规则时默认允许访问。

安装时生成的配置监听 `127.0.0.1:8080`，转发到 `127.0.0.1:80`；对外开放时需要显式修改监听地址。配置文件以 `0600` 权限创建，已有文件不会覆盖。SOCKS5 支持 TCP CONNECT，可以选择开启用户名和密码认证。
