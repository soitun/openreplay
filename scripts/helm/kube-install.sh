#!/bin/bash

set -xo errtrace

# color schemes
# Ansi color code variables
red="\e[0;91m"
blue="\e[0;94m"
expand_bg="\e[K"
blue_bg="\e[0;104m${expand_bg}"
red_bg="\e[0;101m${expand_bg}"
green_bg="\e[0;102m${expand_bg}"
green="\e[0;92m"
white="\e[0;97m"
bold="\e[1m"
uline="\e[4m"
reset="\e[0m"

working_dir=$(pwd)

echo -e ${reset}

## installing kubectl
which kubectl &> /dev/null || {
    echo "kubectl not installed. installing..."
    sudo curl -SsL https://dl.k8s.io/release/v1.20.0/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl ; sudo chmod +x /usr/local/bin/kubectl
}

## installing stern
which stern &> /dev/null || {
    echo "stern not installed. installing..."
    sudo curl -SsL https://github.com/derdanne/stern/releases/download/2.1.16/stern_linux_amd64 -o /usr/local/bin/stern ; sudo chmod +x /usr/local/bin/stern
}

## installing k9s
which k9s &> /dev/null || {
    echo "k9s not installed. installing..."
    sudo curl -SsL https://github.com/derailed/k9s/releases/download/v0.24.2/k9s_Linux_x86_64.tar.gz -o /tmp/k9s.tar.gz
    cd /tmp
    tar -xf k9s.tar.gz
    sudo mv k9s /usr/local/bin/k9s
    sudo chmod +x /usr/local/bin/k9s
    cd -
}

which ansible &> /dev/null || {
    echo "ansible not installed. Installing..."
    which pip || (sudo apt update && sudo apt install python3-pip -y)
    sudo pip3 install ansible==2.10.0
}

which docker &> /dev/null || {
    echo "docker is not installed. Installing..."
    user=`whoami`
    sudo apt install docker.io -y
    sudo usermod -aG docker $user
}

## installing helm
which helm &> /dev/null
if [[ $? -ne 0 ]]; then
    echo "helm not installed. installing..."
    curl -ssl https://get.helm.sh/helm-v3.4.2-linux-amd64.tar.gz -o /tmp/helm.tar.gz
    tar -xf /tmp/helm.tar.gz
    chmod +x linux-amd64/helm
    sudo cp linux-amd64/helm /usr/local/bin/helm
    rm -rf linux-amd64/helm /tmp/helm.tar.gz
fi

# ## checking kubernetes access
# kubectl cluster-info &> /dev/null
# if [[ $? -ne 0 ]]; then
#     echo -e "${red}kubernetes cluster is not accessible.\nplease check ${bold}kubeconfig${reset}${red} env variable is set or ${bold}~/.kube/config exists.${reset}"
#     exit 1
# fi

# make all stderr red
color()(set -o pipefail;"$@" 2>&1>&3|sed $'s,.*,\e[31m&\e[m,'>&2)3>&1

usage()
{
echo -e ${bold}${yellow} '''
This script will install and configure openreplay apps and databases on the kubernetes cluster,
which is accesd with the ${HOME}/.kube/config or $KUBECONFIG env variable.
'''
cat << EOF
▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄
█░▄▄▀█░▄▄█░▄▄▀█░██░█░▄▄█░▄▄▀██
█░▀▀░█▄▄▀█░▀▀░█░▀▀░█░▄▄█░▀▀▄██
█▄██▄█▄▄▄█▄██▄█▀▀▀▄█▄▄▄█▄█▄▄██
▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
EOF
  echo -e "${green}Usage: openreplay-cli [ -h | --help ]
                  [ -v | --verbose ]
                  [ -a | --app APP_NAME ] to install/reinstall specific application
                  [ -t | --type small|medium|ideal ]"
  echo -e "${reset}${blue}type defines the resource limits applied for the installation:
  small: 4core 8G machine
  medium: 4core 16G machine
  ideal: 8core 32G machine

apps can specifically be installed/reinstalled: 
  alerts assets auth chalice ender http integrations ios-proxy metadata negative pg-stateless pg preprocessing redis sink storage frontend
  ${reset}"
  echo type value: $installation_type
  exit 0
}

# Defaults to minimum installation
installation_type=1
type() {
    case "$1" in
        small) installation_type=1 ;;
        medium) installation_type=1.5 ;;
        ideal) installation_type=2 ;;
        *) 
            echo -e ${red}${bold}'ERROR!!!\nwrong value for `type`'${reset}
            usage ;;
    esac
}

function app(){
    case $1 in
        nginx)
            [[ NGINX_REDIRECT_HTTPS -eq 0 ]] && {
                sed -i "/return 301/d" nginx-ingress/nginx-ingress/templates/configmap.yaml
            }
            ansible-playbook -c local setup.yaml -e @vars.yaml -e scale=$installation_type --tags nginx -v
            exit 0
            ;;
        frontend)
            ansible-playbook -c local setup.yaml -e @vars.yaml -e scale=$installation_type --tags frontend -v
            exit 0
            ;;
        *)
            ansible-playbook -c local setup.yaml -e @vars.yaml -e scale=$installation_type -e app_name=$1 --tags app -v
            exit 0
            ;;
    esac
}
# Parsing command line args.
PARSED_ARGUMENTS=$(color getopt -a -n openreplay-cli -o vht:a: --long verbose,help,type:,app: -- "$@")
VALID_ARGUMENTS=$?
if [[ "$VALID_ARGUMENTS" != "0" ]]; then
  usage
fi

eval set -- "$PARSED_ARGUMENTS"
while :
do
  case "$1" in
    -v | --verbose) VERBOSE=1    ; shift   ;;
    -h | --help)    usage        ; shift   ;;
    -t | --type)    type $2    ; shift 2   ;;
    -a | --app)    app $2    ; shift 2   ;;
    # -- means the end of the arguments; drop this, and break out of the while loop
    --) shift; break ;;
    # If invalid options were passed, then getopt should have reported an error,
    # which we checked as VALID_ARGUMENTS when getopt was called...
    *) echo "Unexpected option: $1 - this should not happen."
       usage ;;
  esac
done

[[ $VERBOSE -eq 1 ]] && set -x

{
    ansible-playbook -c local setup.yaml -e @vars.yaml -e scale=$installation_type --tags pre-check -v
} || exit $?
{
    ansible-playbook -c local setup.yaml -e @vars.yaml -e scale=$installation_type --skip-tags pre-check -v
} || exit $?
