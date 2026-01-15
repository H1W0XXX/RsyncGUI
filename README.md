# Rsync GUI（Go + React）

一个简单的图形界面，用 rsync 拉取/推送文件，支持本机-远程,远程-远程 之间同步。

## 快速开始
- 准备 `hosts.yaml`，填写各远程主机的 host/port/user/key（示例见仓库根目录）。（注意linux和windows的私钥路径斜杠）
- 前端：在 `web/` 里 `npm install && npm run dev`；构建产物 `npm run build` 输出到 `web/dist`。
- 运行：hosts.yaml 文件和 可执行文件 rsyncgui-windows-amd64.exe 在同一个目录下，打开电脑浏览器 http://127.0.0.1:8901/ 开始传输文件
## 已知局限
- 未在 macOS 上跑过完整测试。
- SSH 密码登录尚未实测，优先使用私钥登录。
- 依赖远程主机具备 `python3`（用于获取目录）与 `rsync`（传输）。
