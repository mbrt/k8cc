FROM centos:7

USER root
RUN yum install -y --nogpg distcc distcc-server gcc && \
    yum clean all

RUN useradd distcc
ENV DISTCCD_PATH=/cm/local/apps/gcc/6.3.0/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
COMMAND ["distccd"]
