# rlpa-server

Another go implement of estk.me [rlpa-server.php](https://github.com/estkme-group/lpac/blob/main/src/rlpa-server.php)

- [x] Download Profile (not tested yet)
- [x] Process Notification (consistent with the behavior of `rlpa-server.php`, notifications for non "delete" operations will be removed after processing)

Under construction:

- Remote Management(http api)

This feature is currently under construction. However, it is not necessary to use this feature in normal usage scenarios, and I am not sure if I should complete it

## Usage

Compile latest [lpac](https://github.com/estkme-group/lpac), then place the `lpac` binary program in the same directory as the `rlpa-server` program

use environment variables to set port

- `SOCKET_PORT`: socket port for estk rlpa, default 1888
- `API_PORT`: http management api port, default 8008

debug log output: start with `-debug` argument to enable debug log level

### systemd service example

Write the following content into `/etc/systemd/system/rlpa-server.service`
```
[Unit]
Description=rlpa server
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=/path/to/rlpa-server
WorkingDirectory=/path/to/rlpa-server-directory
Restart=on-failure
User=[your-user]

[Install]
WantedBy=multi-user.target
```
- `ExecStart`: rlpa-server binary path
- `WorkingDirectory`: The directory where rlpa server is located
- `User`: Replace with actual user

Then execute `sudo systemctl daemon-reload` to reload services

- Start rlpa-server: `sudo systemctl start rlpa-server`
- Let rlpa-server start with system: `sudo systemctl enable rlpa-server`

## Public Server
⚠️ No guarantee, use at your own risk

|      IP      | Port |     Location      |
|:------------:|:----:|:-----------------:|
|205.185.117.85| 1888 | Las Vegas, NV, US |


## API Document

- lpac shell command

Post json to `/shell/{manageID}` with header `Password: {Password}`

example

```bash
curl -X POST -H "Content-Type: application/json" \
-H "Password: 2660" \
-d '{"type":0, "command":"chip info"}' \
http://example.com:8008/shell/rAct
```

Will get lpac output