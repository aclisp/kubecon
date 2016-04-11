FROM 61.160.36.122:8080/lightvm:latest

# 安装进程可执行文件（由 main.go 编译）
COPY kubecon /

# 设置自动拉起进程
RUN mkdir /etc/service/kubecon
COPY entrypoint.sh /etc/service/kubecon/run
RUN chmod +x /etc/service/kubecon/run

# 安装资源文件
COPY css /var/lib/kubecon/css
COPY js /var/lib/kubecon/js
COPY fonts /var/lib/kubecon/fonts
COPY img /var/lib/kubecon/img
COPY pages /var/lib/kubecon/pages

# The entrypoint of lightvm will start everything
# under `/etc/service` as daemon
