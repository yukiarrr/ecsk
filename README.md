[English](https://github.com/yukiarrr/ecsk/blob/main/README.md) / [日本語](https://github.com/yukiarrr/ecsk/blob/main/README.ja.md)

# ecsk

**EC**S + Ta**sk** = **ecsk** 😆

ecsk is a CLI tool to interactively use frequently used functions of docker command in Amazon ECS.  
(docker run, exec, cp, logs, stop)

![ecsk](https://github.com/yukiarrr/ecsk/raw/main/docs/images/ecsk.gif)

It specializes in handling a single container (task) like docker command, not an orchestration.

## Install

### MacOS

```sh
brew install yukiarrr/tap/ecsk
```

### Linux

```sh
wget https://github.com/yukiarrr/ecsk/releases/download/v0.5.3/ecsk_Linux_x86_64.tar.gz
tar zxvf ecsk_Linux_x86_64.tar.gz
chmod +x ./ecsk
sudo mv ./ecsk /usr/local/bin/ecsk
```

## Usage

Here are some frequently used commands.  
For detailed flags, run `ecsk [command] --help` to check them.

### `ecsk run`

```sh
ecsk run
```

If you don't specify any flags, after entering task information interactively, the log will continue to flow until the task is started and stopped as in `docker run`.
<br>
<br>

```sh
ecsk run -e -i --rm -c [container_name] -- /bin/sh
```

After the task is started, execute the command specified by `execute-command`.  
By specifying `--rm`, the task will be automatically stopped at the end of the session, so you can operate it like a bastion host.
<br>
<br>

```sh
ecsk run -d
```

After entering the task information interactively, the command will be stopped without waiting for the task to start or stop.

### `ecsk exec`

```sh
ecsk exec -i -- /bin/sh
```

After selecting the task and container interactively, and execute the command.

### `ecsk cp`

```sh
ecsk cp ./ [container_name]:/etc/nginx/
```

After selecting the task interactively, copy the files from local to remote.  
Internally, using an S3 Bucket to transfer the files, [so you need to add permissions for the corresponding Bucket to the task role.](#When-using-ecsk-cp)

If you want to select the container interactively, use `ecsk cp . / :/etc/nginx/`.
<br>
<br>

```sh
ecsk cp [container_name]:/var/log/nginx/access.log ./
```

Transfer files from remote to local.

### `ecsk logs`

```sh
ecsk logs
```

After selecting the task interactively, view logs.  
Multiple tasks can be specified.

ecsk uses [knqyf263/utern](https://github.com/knqyf263/utern) to view logs.

### `ecsk stop`

```sh
ecsk stop
```

After selecting the task interactively, stop.

## Prerequisites

### When using `ecsk exec`

Since ecsk is executing `execute-command` internally, there are some prerequisites.  
Here are the prerequisites with reference to [the official documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html).

#### Install Session Manager plugin

Please refer to the following.

https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html

#### Prmissions required for ECS Exec

The ECS Exec feature requires a task IAM role to grant containers the permissions needed for communication between the managed SSM agent (execute-command agent) and the SSM service.

```json
{
   "Version": "2012-10-17",
   "Statement": [
       {
       "Effect": "Allow",
       "Action": [
            "ssmmessages:CreateControlChannel",
            "ssmmessages:CreateDataChannel",
            "ssmmessages:OpenControlChannel",
            "ssmmessages:OpenDataChannel"
       ],
      "Resource": "*"
      }
   ]
}
```

#### Enabling ECS Exec

You need to enable ECS Exec in order to `execute-command` on a task of a service that has already been created.  
Add the `--enable-execute-command` flag for AWS CLI, or `EnableExecuteCommand` for CFn.

Note that you should use the `-e` or `--enable-execute-command` flag for tasks started with `ecsk run`.

#### Supplement

As these are more prerequisites, ecsk will run [aws-containers/amazon-ecs-exec-checker](https://github.com/aws-containers/amazon-ecs-exec-checker) on errors.

### When using `ecsk cp`

Since ecsk uses S3 Bucket for file transfer, you need to add permissions for the corresponding bucket to the task role.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket",
                "s3:GetObject",
                "s3:PutObject",
                "s3:PutObjectAcl"
            ],
            "Resource": [
                "arn:aws:s3:::[bucket_name]",
                "arn:aws:s3:::[bucket_name]/ecsk_*"
            ]
        }
    ]
}
```
