FROM centos:7
MAINTAINER <cesec>

# copy version
RUN mkdir -p /opt/ygobuster 
COPY VERSION /opt/ygobuster

ADD gobuster /opt/ygobuster
RUN chmod +x /opt/ygobuster/gobuster
ADD wordlist.txt /opt/ygobuster
WORKDIR /opt/ygobuster

ENTRYPOINT ["./ygobuster"]
