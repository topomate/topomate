FROM frrouting/frr:7.3.1

ADD daemons /etc/frr/daemons
ADD vtysh.conf /etc/frr/vtysh.conf

RUN apk add iperf3 &&\
    apk add tcpdump