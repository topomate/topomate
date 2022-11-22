FROM frrouting/frr:v8.4.1

RUN apk add iperf3 &&\
    apk add tcpdump &&\
    apk add tcptraceroute &&\
    apk add busybox-extras &&\
    apk add python3

RUN ln -s /usr/bin/python3 /usr/bin/python

RUN touch /var/log/frr.log && chown frr:frr /var/log/frr.log

ADD daemons /etc/frr/daemons
ADD vtysh.conf /etc/frr/vtysh.conf
ADD delayed-reload /usr/bin/delayed-reload
RUN chmod +x /usr/bin/delayed-reload

COPY docker-start /usr/sbin/docker-start
RUN chmod +x /usr/sbin/docker-start
ENTRYPOINT ["/usr/sbin/docker-start"]