FROM alpine/git:latest
COPY git-mirror-linux-amd64 /bin/git-mirror
ENTRYPOINT ["/bin/git-mirror"]
