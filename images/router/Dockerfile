FROM frrouting/frr:latest

ADD daemons /etc/frr/daemons
ADD vtysh.conf /etc/frr/vtysh.conf

RUN apk add iperf3 &&\
    apk add tcpdump