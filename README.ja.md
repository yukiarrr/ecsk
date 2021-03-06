[English](https://github.com/yukiarrr/ecsk/blob/main/README.md) / [æ¥æ¬èª](https://github.com/yukiarrr/ecsk/blob/main/README.ja.md)

# ecsk

**EC**S + Ta**sk** = **ecsk** ð

ecskã¯ãã¤ã³ã¿ã©ã¯ãã£ãã«Amazon ECS APIï¼run-task, execute-command, stop-taskï¼ãå¼ã³åºããããECSã¨ã­ã¼ã«ã«ã®éã§ãã¡ã¤ã«ãã³ãã¼ããããã­ã°ãè¡¨ç¤ºãããã§ããCLIãã¼ã«ã§ãã

![ecsk](https://github.com/yukiarrr/ecsk/raw/main/docs/images/ecsk.gif)

ecskã¯ã³ã³ããï¼ã¿ã¹ã¯ï¼ãåãæ±ããã¨ã«ç¹åãã¦ããã®ã§ã

- ECSãµã¼ãã¹ãã¿ã¹ã¯å®ç¾©ã®ç®¡ç â CDKãTerraformãªã©ãä½¿ç¨
- ãããã° â **ecsk**ãä½¿ç¨ ð

ãªã©ã®å©ç¨ãæ³å®ãã¦ãã¾ãã

## ã¤ã³ã¹ãã¼ã«

### MacOS

```sh
brew install yukiarrr/tap/ecsk
```

### Linux

```sh
wget https://github.com/yukiarrr/ecsk/releases/download/v0.7.0/ecsk_Linux_x86_64.tar.gz
tar zxvf ecsk_Linux_x86_64.tar.gz
chmod +x ./ecsk
sudo mv ./ecsk /usr/local/bin/ecsk
```

### Windows

[Releases](https://github.com/yukiarrr/ecsk/releases)ãããã¦ã³ã­ã¼ããã¦ãã ããã

## ä½¿ãæ¹

ããã§ã¯ãããä½¿ãã³ãã³ããç´¹ä»ãã¾ãã  
è©³ãããã©ã°ã«ã¤ãã¦ã¯ã`ecsk [command] --help`ãå®è¡ãã¦ç¢ºèªãã¦ãã ããã

### `ecsk run`

```sh
ecsk run
```

ãã©ã°ãä¸åæå®ããªãå ´åã¯ãã¤ã³ã¿ã©ã¯ãã£ãã«ã¿ã¹ã¯æå ±ãå¥åããå¾ã`docker run`ã¨åãããã«ã¿ã¹ã¯ãèµ·åãçµäºããã¾ã§ã­ã°ãæµãç¶ãã¾ãã
<br>
<br>

```sh
ecsk run -e -i --rm -c [container_name] -- /bin/sh
```

ã¿ã¹ã¯ãèµ·åãããå¾ã`execute-command`ã§æå®ããã³ãã³ããå®è¡ãã¾ãã  
åããã¦`--rm`ãæå®ãããã¨ã§ãã»ãã·ã§ã³çµäºæã«èªåã§ã¿ã¹ã¯ãçµäºãããããè¸ã¿å°ãµã¼ãã¼ã®ããã«éç¨ãããã¨ãå¯è½ã«ãªãã¾ãã
<br>
<br>

```sh
ecsk run -d
```

ã¤ã³ã¿ã©ã¯ãã£ãã«ã¿ã¹ã¯æå ±ãå¥åããå¾ãã¿ã¹ã¯ã®éå§ã»çµäºãå¾ããã«ã³ãã³ããçµäºããã¾ãã

### `ecsk exec`

```sh
ecsk exec -i -- /bin/sh
```

ã¤ã³ã¿ã©ã¯ãã£ãã«ã¿ã¹ã¯ã»ã³ã³ãããé¸æããã³ãã³ããå®è¡ãã¾ãã

### `ecsk cp`

```sh
ecsk cp ./ [container_name]:/etc/nginx/
```

ã¤ã³ã¿ã©ã¯ãã£ãã«ã¿ã¹ã¯ãé¸æããã­ã¼ã«ã«ãããªã¢ã¼ãã¸ãã¡ã¤ã«ãã³ãã¼ãã¾ãã  
åé¨çã«ã¯S3 Bucketãä½¿ç¨ãã¦ãã¡ã¤ã«ãè»¢éãã¦ããã®ã§ã[ã¿ã¹ã¯ã­ã¼ã«ã«è©²å½Bucketã®ã¢ã¯ã»ã¹è¨±å¯ãè¿½å ããå¿è¦ãããã¾ãã](#ecsk-cpãä½¿ãå ´å)

ãªããã³ã³ãããã¤ã³ã¿ã©ã¯ãã£ãã«é¸æããå ´åã¯ã`ecsk cp ./ :/etc/nginx/`ã¨ãã¦ãã ããã
<br>
<br>

```sh
ecsk cp [container_name]:/var/log/nginx/access.log ./
```

ãªã¢ã¼ãããã­ã¼ã«ã«ã«ãã¡ã¤ã«ãè»¢éãã¾ãã

### `ecsk logs`

```sh
ecsk logs
```

ã¤ã³ã¿ã©ã¯ãã£ãã«ã¿ã¹ã¯ãé¸æããã­ã°ãè¡¨ç¤ºãã¾ãã  
ãã®ã¿ã¹ã¯ã¯è¤æ°æå®ãããã¨ãã§ãã¾ãã

ãªããã­ã°è¡¨ç¤ºã¯[knqyf263/utern](https://github.com/knqyf263/utern)ãå©ç¨ããã¦ããã ãã¦ãã¾ãã

### `ecsk stop`

```sh
ecsk stop
```

ã¤ã³ã¿ã©ã¯ãã£ãã«ã¿ã¹ã¯ãé¸æããçµäºãã¾ãã

### `ecsk describe`

```sh
ecsk describe
```

ã¤ã³ã¿ã©ã¯ãã£ãã«ã¿ã¹ã¯ãé¸æããè©³ç´°æå ±ãè¡¨ç¤ºãã¾ãã  
ã¾ããã¿ã¹ã¯ä¸è¦§ãç¢ºèªããç¨éã¨ãã¦ãå©ç¨ã§ãã¾ãã

## åææ¡ä»¶

### `ecsk exec`ãä½¿ãå ´å

åé¨ã§`execute-command`ãå®è¡ãã¦ãããããããã¤ãã®åææ¡ä»¶ãããã¾ãã  
ããã§ã¯ã[å¬å¼ãã­ã¥ã¡ã³ã](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/ecs-exec.html)ãåèã«ãå¿é é ç®ãç´¹ä»ãã¾ãã

#### Session Manager pluginãã¤ã³ã¹ãã¼ã«

ä¸è¨ãåèã«ãã¦ãã ããã

https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html

#### SSMã®ã¢ã¯ã»ã¹è¨±å¯ãè¿½å 

SSMã¨ã¼ã¸ã§ã³ãã¨SSMãµã¼ãã¹éã®éä¿¡ã«å¿è¦ãªã¢ã¯ã»ã¹è¨±å¯ãè¿½å ããå¿è¦ãããã¾ãã

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

#### ECS Execã®æå¹å

ãã§ã«ä½æããã¦ãããµã¼ãã¹ã®ã¿ã¹ã¯ã§`execute-command`ããããã«ã¯ãECS Execãæå¹åããå¿è¦ãããã¾ãã  
AWS CLIã§ããã°`--enable-execute-command`ãã©ã°ããCFnã§ããã°`EnableExecuteCommand`ãè¿½å ãã¦ãã ããã

ãªãã`ecsk run`ã§èµ·åããã¿ã¹ã¯ã«é¢ãã¦ã¯ã`-e`ã`--enable-execute-command`ãã©ã°ãä½¿ç¨ãã¦ãã ããã

#### è£è¶³

ãããã®ããã«ãåææ¡ä»¶ãå¤ãã¨ãªã£ã¦ããã®ã§ãecskã§ã¯ã¨ã©ã¼æã«[aws-containers/amazon-ecs-exec-checker](https://github.com/aws-containers/amazon-ecs-exec-checker)ãå®è¡ããããã«ãã¦ãã¾ãã

### `ecsk cp`ãä½¿ãå ´å

ãã¡ã¤ã«ã®åãæ¸¡ãã«S3 Bucketãç¨ãã¦ãããããã¿ã¹ã¯ã­ã¼ã«ã«è©²å½Bucketã®ã¢ã¯ã»ã¹è¨±å¯ãè¿½å ããå¿è¦ãããã¾ãã

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
