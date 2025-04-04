FROM golang:latest AS builder

WORKDIR /opt

# 需要先运行 go mod vendor
# 避免在构建镜像时下载依赖
# 顺带加速构建镜像
COPY . .

# 构建web程序
RUN go build -ldflags="-s -w" -o web .

FROM ubuntu:latest

WORKDIR /opt

# 最终镜像仅保留必要的文件
COPY --from=builder /opt/web /opt/web

COPY --from=builder /opt/config.ini /opt/config.ini

COPY --from=builder /opt/country_asn.mmdb /opt/country_asn.mmdb

COPY --from=builder /opt/html /opt/html

COPY --from=builder /opt/template /opt/template

COPY --from=builder /opt/salt /opt/salt

# TODO:tls证书不打包进容器
COPY --from=builder /opt/cert.pem /opt/cert.pem

COPY --from=builder /opt/key.pem /opt/key.pem

# 开放443端口
EXPOSE 443

CMD ["/opt/web"]