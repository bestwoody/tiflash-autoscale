if [[ $# -eq 0 ]] ; then
    echo 'need args!'
    exit 1
fi
docker build -t bestwoody/k8stest:$1 . && docker push bestwoody/k8stest:$1
