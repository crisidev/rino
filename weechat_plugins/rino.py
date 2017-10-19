import os
import socket
import datetime
import weechat


SCRIPT_NAME = 'rino'
SCRIPT_AUTHOR = 'Matteo Bigoi <bigo@crisidev.org>'
SCRIPT_VERSION = '0.1'
SCRIPT_LICENSE = 'MIT'
SCRIPT_DESC = 'Pass highlights and private messages to the OS X 10.8+ Notification Center'

weechat.register(SCRIPT_NAME, SCRIPT_AUTHOR, SCRIPT_VERSION, SCRIPT_LICENSE, SCRIPT_DESC, '', '')

DEFAULT_OPTIONS = {
    'show_highlights': 'on',
    'show_private_message': 'on',
    'show_message_text': 'on',
    'ignore_old_messages': 'off',
    'server_addr': '127.0.0.1',
    'msg_separator': '|!|'
}

for key, val in DEFAULT_OPTIONS.items():
    if not weechat.config_is_set_plugin(key):
        weechat.config_set_plugin(key, val)

weechat.hook_print('', 'irc_privmsg', '', 1, 'notify', '')


def push_notification(user, message):
    rino_dir = os.path.join(weechat.info_get("weechat_dir", ""), "rino")
    clients = os.listdir(rino_dir)

    if len(clients) > 0:
        tag, port = clients[0].split(":")
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            s.connect((weechat.config_get_plugin('server_addr'), int(port)))
            s.send(user + weechat.config_get_plugin('msg_separator') + message + '\n')
            s.close()
        except Exception as e:
            weechat.prnt("error sending rino notification: %s", e.message)


def notify(data, buffer, date, tags, displayed, highlight, user, message):
    # ignore if it's yourself
    own_nick = weechat.buffer_get_string(buffer, 'localvar_nick')
    if user == own_nick or user == ('@%s' % own_nick):
        return weechat.WEECHAT_RC_OK

    if weechat.config_get_plugin('ignore_old_messages') == 'on':
        message_time = datetime.datetime.utcfromtimestamp(int(date))
        now_time = datetime.datetime.utcnow()

        # ignore if the message is greater than 5 seconds old
        if (now_time - message_time).seconds > 5:
            return weechat.WEECHAT_RC_OK

    if weechat.config_get_plugin('show_message_text') == 'off':
        message = "Private message"

    if weechat.config_get_plugin('show_highlights') == 'on' and int(highlight):
        channel = weechat.buffer_get_string(buffer, 'localvar_channel')
        user = user + '@' + channel
        push_notification(user, message)
    elif weechat.config_get_plugin('show_private_message') == 'on' and 'notify_private' in tags:
        push_notification(user, message)

    return weechat.WEECHAT_RC_OK
