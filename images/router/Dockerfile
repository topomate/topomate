FROM frrouting/frr:v7.3.1

RUN apk add iperf3 &&\
    apk add tcpdump &&\
    apk add busybox-extras

ADD daemons /etc/frr/daemons
ADD vtysh.conf /etc/frr/vtysh.conf

COPY docker-start /usr/sbin/docker-start
RUN chmod +x /usr/sbin/docker-start
ENTRYPOINT ["/usr/sbin/docker-start"]