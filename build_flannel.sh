#!/bin/bash
make clean dist/flanneld-amd64
eval $(aws ecr get-login --profile adparts_docker_registry | sed 's|https://||' | sed 's|-e none||')
docker build -f Dockerfile.amd64 -t 505717776300.dkr.ecr.eu-west-1.amazonaws.com/adparts:flanneld-autorestart-0.9.1 .
docker push 505717776300.dkr.ecr.eu-west-1.amazonaws.com/adparts:flanneld-autorestart-0.9.1

docker login -u indiketa 
docker tag 505717776300.dkr.ecr.eu-west-1.amazonaws.com/adparts:flanneld-autorestart-0.9.1 indiketa/flanneld-autorestart:0.9.1
docker push indiketa/flanneld-autorestart:0.9.1
