
## Backend Agent Service

### The endpoints ready are:

- /api/v1/login (Login)

            {
                "username":"duser",
                "password":"letmepass"
            }

- /api/v1/campaign/login

           {
               "username": "duser",
               "campaign": "dcamp"
            }

- /api/v1/logout

            {
                "username": "duser"
            }

- /api/v1/closer/inbound
            {
                "username": "duser",
                "groups": ["option1", "option2", "option3"],
                "phone": "100",
                "campaign": "dcamp",
                "blended": "true"
            }

- /api/v1/dial/manual

           {
               "phone_number":"722116291",
               "username": "duser",
               "phone": "100",
               "campaign": "dcamp",
               "list_id": "998"
            }


- /api/v1/dial/deleteCallback

            {
                "username": "duser",
                "phone": "1000",
                "campaign": "dcamp",
                "callbackID": "122323"
            };
- /api/v1/dial/hangup (Hangup a call)

           {
               "username": "duser",
               "phone": "100",
               "campaign": "dcamp",
               "lead_id": "123",
               "callerid": "998",
               "phone_number":"722116291",
            }

- /api/v1/dial/dispose (Dispose a call )

           {
               "username": "duser",
               "phone"   : "100",
               "campaign": "dcamp",
               "lead_id" : "lead_id",
               "status": "sale",
               "call_type":"IN"
            }

- /api/v1/dial/next
- /api/v1/dial/park
- /api/v1/dial/grab
- /api/v1/dial/take

           `{
               "username": "duser",
               "phone":    "100",
               "campaign": "dcamp",
               "lead_id":  "45",
               "call_id": "2344",
               "phone_number": "722116291"
           }`

- /api/v1/dial/dtmf
- /api/v1/dial/status
- /api/v1/dial/pause-code-switch
- /api/v1/dial/logs (See all the logs of an Individual Agent)

           {
               "campaign":99999,
               "user": "duser",
               "phone":"100",
               "date" : "2020-03-22"
            }

- /api/v1/transfer/local
- /api/v1/transfer/add
- /api/v1/transfer/park
- /api/v1/transfer/hangall
- /api/v1/transfer/hangxfer
- /api/v1/transfer/hang-customer
- /api/v1/transfer/blind



## Common commands


- make build && scp smartdial  root@172.16.10.204:backend

make build && scp smartdial  root@192.168.1.244:backend
- cat storage/logs/smartdial.log


## Deployment process

- In the root directory, change directory by running the command  `cd /etc/systemd/system`

- Create a file and name it smartdial.service using the command `touch smartdial.service`

- Open the smartdial.service file using the command `nano smartdial.service`

- Paste the following code

    `[Unit]`
    `Description=smartdial`

    `[Service]`
    `WorkingDirectory=/root/backend`__
    `Type=simple`__
    `Restart=always`__
    `RestartSec=5s`__
    `ExecStart=/root/backend/smartdial s`

    `[Install]`
    `WantedBy=multi-user.target`

- Enable the service using the command `systemctl enable smartdial`

- Start the service using the command `sudo service smartdial start`

- If need be,you can stop the service using the command `sudo service smartdial stop`

- Check the status of the service using the command `sudo service smartdial status`

