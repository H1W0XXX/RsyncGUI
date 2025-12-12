# Rsync GUI（Go + React）

一个简单的图形界面，用 rsync 拉取/推送文件，支持本机-远程,远程-远程 之间同步。

## 快速开始
- 准备 `hosts.yaml`，填写各远程主机的 host/port/user/key（示例见仓库根目录）。
- 运行：`./rsyncgui`（可用环境变量调整）
- 前端：在 `web/` 里 `npm install && npm run dev`；构建产物 `npm run build` 输出到 `web/dist`。

## 已知局限
- 未在 Windows/macOS 上跑过完整测试。
- SSH 密码登录路径尚未实测，优先使用私钥登录。
- 依赖远程主机具备 `python3`（用于列目录）与 `rsync`（传输）。
