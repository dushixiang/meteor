# Meteor

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
geoip:
  file: GeoLite2-City.mmdb    # 配置geoip后支持按城市配置规则
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
  forward     Forward the received data to the destination address
  help        Help about any command
  install     Install meteor as a system service
  proxy       Start a proxy server
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