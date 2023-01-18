#docker build -t bestwoody/tiflash-autoscaler:serverless.v6.4.0 . && docker push bestwoody/tiflash-autoscaler:serverless.v6.4.0
docker buildx build --platform=linux/amd64,linux/arm64 -t bestwoody/tiflash-autoscaler:serverless.v6.4.0 .  --push
