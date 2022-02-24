# Telegram TOTP Bot

A Telegram Bot for managing (registering, deleting, generating tokens) TOTP.

## Installation

```bash
$ git clone https://github.com/meinside/telegram-totp-bot.git
$ cd telegram-totp-bot/
$ go build
```

## Configuration

Create a config file:

```json
{
    "telegram_bot_token": "YOUR_TELEGRAM_BOT_TOKEN",
    "database_file_location": "/path/to/your/database.db"
}
```

## Run

and then run with:

```bash
$ ./telegram-totp-bot -config /path/to/your/config.json
```

## Or run with systemd

Create a systemd service file:

```
[Unit]
Description=Telegram TOTP Bot
After=syslog.target
After=network.target

[Service]
Type=simple
User=ubuntu
Group=ubuntu
WorkingDirectory=/path/to/your/telegram-totp-bot-directory/
ExecStart=/path/to/your/telegram-totp-bot-directory/telegram-totp-bot -config /path/to/your/config.json
Restart=always
RestartSec=5
Environment=
MemoryAccounting=true
MemoryHigh=100M
MemoryMax=128M

[Install]
WantedBy=multi-user.target
```

at `/lib/systemd/system/telegram-totp-bot.service`,

and run:

```bash
# make it launch automatically at boot
$ systemctl enable telegram-totp-bot.service

# start, restart, or stop it
$ systemctl start telegram-totp-bot.service
$ systemctl restart telegram-totp-bot.service
$ systemctl stop telegram-totp-bot.service
```

## License

MIT

