# Amazon Web Services

## Introduction

This project allows you to get hold of some machine on Amazon Web Services.
You can then use these machines as is or run various Ansible playbooks from `../config_management` to set up Weave Net, Kubernetes, etc.

## Setup

* Log in [weaveworks.signin.aws.amazon.com/console](https://weaveworks.signin.aws.amazon.com/console/) with your account.

* Go to `Services` > `IAM` > `Users` > Click on your username > `Security credentials` > `Create access key`.
  Your access key and secret key will appear on the screen. Set these as environment variables:

```
export AWS_ACCESS_KEY_ID=<your access key> 
export AWS_SECRET_ACCESS_KEY=<your secret key>
```

* Go to `Services` > `EC2` > Select the availability zone you want to use (see top right corner, e.g. `us-east-1`) > `Import Key Pair`.
  Enter your SSH public key and the name for it, and click `Import`.
  Set the path to your private key as an environment variable:

```
export TF_VAR_aws_public_key_name=<your Amazon Web Services SSH key name>
export TF_VAR_aws_private_key_path="$HOME/.ssh/id_rsa"
```

* Set your current IP address as an environment variable:

```
export TF_VAR_client_ip=$(curl -s -X GET http://checkip.amazonaws.com/)
```

  or pass it as a Terraform variable:

```
$ terraform <command> -var 'client_ip=$(curl -s -X GET http://checkip.amazonaws.com/)'
```

## Usage

* Create the machine: `terraform apply`
* Show the machine's status: `terraform show`
* Stop and destroy the machine: `terraform destroy`
* SSH into the newly-created machine:

```
$ ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no `terraform output username`@`terraform output public_ips`
# N.B.: the default username will differ depending on the AMI/OS you installed, e.g. ubuntu for Ubuntu, ec2-user for Red Hat, etc.
```

## Resources

* [https://www.terraform.io/docs/providers/aws/](https://www.terraform.io/docs/providers/aws/)
* [https://www.terraform.io/docs/providers/aws/r/instance.html](https://www.terraform.io/docs/providers/aws/r/instance.html)
* [Terraform variables](https://www.terraform.io/intro/getting-started/variables.html)
