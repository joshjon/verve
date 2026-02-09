FROM alpine:3.19

RUN apk add --no-cache bash

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV TASK_ID=""
ENV TASK_DESCRIPTION=""

ENTRYPOINT ["/entrypoint.sh"]
