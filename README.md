go-forwarder
=============
**ss-redir** 转发客户端的请求到远程的 **ss-server**,但是在只有socks5代理的情况下，**ss-redir** 就无法使用。
使用本程序可以将请求转发给socks5 server,完成透明的数据转发

使用方法
========
./go-fowarder listen addr:port -socks5 remoteaddr:port

如何在在PC上搭建透明代理
======================
https://alex8224.github.io/2018/05/30/build-tproxy/


