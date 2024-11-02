postgrest

https://postgrest.org/en/v12/tutorials/tut1.html


jwt: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoidG9kb191c2VyIn0.7TPzLmz4OB4NFqvEefcAKOZKbUIIDF-y-beD2Q2peQg


https://blackpine.io/posts/2022.04.15-using-pull-through-image-registry-with-k3d/
using a local image cache for the docker containers.

https://community.grafana.com/t/public-dashboard-grafana-in-external-link-without-login-tutorial/59221
k3d image import rtop --cluster wb


root@monitor-57967545dc-n49jk:/opt/shell-tutor# cat /opt/checkStorage.sh 
#!/bin/bash

while inotifywait -e modify /opt/storage; do 
  ls /opt/storage

done

k3d image import shell-tutor:latest  --cluster wb

