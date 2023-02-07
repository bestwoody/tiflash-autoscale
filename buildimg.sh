if [[ $# -eq 0 ]] ; then
    echo 'some message'
    exit 1
fi
docker build -t bestwoody/k8stest:$1 . && docker push bestwoody/k8stest:$1
