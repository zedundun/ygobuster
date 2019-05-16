FROM centos:7
MAINTAINER <cesec>

# copy version
RUN mkdir -p /ygobuster 
COPY VERSION /ygobuster

ADD gobuster /ygobuster
RUN chmod +x /ygobuster/gobuster
ADD wordlist.txt /ygobuster
WORKDIR /ygobuster

ENTRYPOINT ["./ygobuster"]
