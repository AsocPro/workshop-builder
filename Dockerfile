FROM panubo/sshd:latest




RUN mkdir -p /home/admin && useradd -d /home/admin admin && chown admin:admin /home/admin && echo 'admin:$6$VePHlvsH0Zimz7xQ$NAeClxVkW2kMUxcDkJ07h3npnaoB7/5g/RBm/grSOILE37Gv/x.IbFpEHoXRCAvQy4QOQSjJWXB1DVIeplrAI0' | chpasswd --encrypted


