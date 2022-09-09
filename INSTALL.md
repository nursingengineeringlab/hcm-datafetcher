# How to run local

### make sure you install golang compiler 

```
Remove any previous Go installation by deleting the /usr/local/go folder (if it exists), then extract the archive you just downloaded into /usr/local, creating a fresh Go tree in /usr/local/go:
$ rm -rf /usr/local/go && tar -C /usr/local -xzf go1.19.1.linux-amd64.tar.gz
(You may need to run the command as root or through sudo).

Do not untar the archive into an existing /usr/local/go tree. This is known to produce broken Go installations.

Add /usr/local/go/bin to the PATH environment variable.
You can do this by adding the following line to your $HOME/.profile or /etc/profile (for a system-wide installation):

export PATH=$PATH:/usr/local/go/bin
Note: Changes made to a profile file may not apply until the next time you log into your computer. To apply the changes immediately, just run the shell commands directly or execute them from the profile using a command such as source $HOME/.profile.

Verify that you've installed Go by opening a command prompt and typing the following command:
$ go version
Confirm that the command prints the installed version of Go.
```

### go build binary

```
go build ./cmd/data-fetcher
```

### config url or run with command line

```
./data-fetcher -mqtt tcp://127.0.0.1:1883  -redis 127.0.0.1:639
```

```
var args struct {
    Mqtt  string `default:"tcp://127.0.0.1:1883"`
    Redis string `default:"127.0.0.1:6379"`
}
```

# how to deploy on kubernetes


### make sure you commit and push the code to codebase

```
git add -A
git commit -m "some changes"
git push origin/main
```

### Docker build local image

```
docker build . -t nelab/hcm-datafetcher:latest
```

### Docker push to dockerhub

```
docker push nelab/hcm-datafetcher:latest
```

### Login to kubernetes cluster

```
ssh nelab.ddns.umass.edu

```

### delete existing pod 

```
kubectl get namespace

NAME              STATUS   AGE
kube-system       Active   186d
kube-public       Active   186d
kube-node-lease   Active   186d
default           Active   186d
monitoring        Active   186d
redis             Active   170d
emqx              Active   169d
postgresql        Active   169d
ingress           Active   166d
traefik-v2        Active   163d
ingress-nginx     Active   163d
app               Active   162d

kubectl ns app  (make sure you install kubectx and kubens plugin)

Context "microk8s" modified.
Active namespace is "app".

kubectl get pods

NAME                           READY   STATUS    RESTARTS       AGE
datafetcher-8568568764-58vcb   1/1     Running   10 (16d ago)   149d
api-64475d566b-bpd7t           1/1     Running   0              8d
frontend-7bf4878444-h8hlv      1/1     Running   0              8d


kubectl delete pod datafetcher-8568568764-58vcb

```

### wait for pod recreate and then it will pull the latest image from docker hub

