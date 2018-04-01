import boto3
import csv
import datetime
import io
import json
import os

from boto3.dynamodb.conditions import Key
from email.mime.text import MIMEText
from email.mime.application import MIMEApplication
from email.mime.multipart import MIMEMultipart

dynamodb = boto3.resource('dynamodb')
ses = boto3.client('ses')

print('Loading function')

def lambda_handler(event, context):
    end = datetime.datetime.now()
    start = end - datetime.timedelta(days=int(event['days']))

    table = dynamodb.Table(os.environ['DYNAMODB_TABLE'])

    fe = Key('date').between(start.isoformat(), end.isoformat())

    response = table.scan(
        FilterExpression=fe,
    )

    with open('/tmp/emails.csv', 'w', newline='') as file:
        writer = csv.writer(file,
                            quoting=csv.QUOTE_NONNUMERIC,
        )

        for i in response['Items']:
            writer.writerow([i['email']])

    msg = MIMEMultipart()
    msg['Subject'] = event['email']['subject']
    msg['From'] = event['email']['from']
    msg['To'] = ', '.join(event['email']['to'])

    part = MIMEText('Attached is your captive portal email report from {} to {}.'.format(start.isoformat(), end.isoformat()))
    msg.attach(part)

    part = MIMEApplication(open('/tmp/emails.csv', 'rb').read())
    part.add_header('Content-Disposition', 'attachment', filename='emails.csv')
    msg.attach(part)

    ses.send_raw_email(
        RawMessage={'Data': msg.as_string()},
        Source=msg['From'],
        Destinations=event['email']['to']
    )
    
