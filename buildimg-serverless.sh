if [[ $# -eq 0 ]] ; then
    echo 'need args!'
    exit 1
fi
#docker build -t bestwoody/tiflash-autoscaler:serverless.v6.4.0 . && docker push bestwoody/tiflash-autoscaler:serverless.v6.4.0
# docker buildx build --platform=linux/amd64,linux/arm64 -t bestwoody/tiflash-autoscaler:serverless.v6.4.0 .  --push
# docker buildx build --platform=linux/amd64,linux/arm64 -t 646528577659.dkr.ecr.us-east-2.amazonaws.com/tiflash-autoscaler:serverless.v6.4.0 .  --push
docker buildx build --platform=linux/amd64,linux/arm64 -t 646528577659.dkr.ecr.us-east-2.amazonaws.com/tiflash-autoscaler:$1 .  --push
