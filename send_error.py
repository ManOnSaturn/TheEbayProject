import os
import smtplib
import sys
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText

from dotenv import load_dotenv

# Configuration
load_dotenv()
sender_email = os.getenv("MAIL")
receiver_email = os.getenv("MAIL")
subject = "Error from Golang scraper"
smtp_server = "smtp.gmail.com"
smtp_port = 587
smtp_username = os.getenv("MAIL")
smtp_password = os.getenv("MAIL_PASSWORD")

def send_email(subject, body, sender, receiver):
    # Create the email
    msg = MIMEMultipart()
    msg['From'] = sender
    msg['To'] = receiver
    msg['Subject'] = subject

    # Attach the body with the msg instance
    msg.attach(MIMEText(body, 'plain'))

    # Create server object with SSL option
    server = smtplib.SMTP(smtp_server, smtp_port)
    server.starttls()

    # Perform operations via server
    server.login(smtp_username, smtp_password)
    text = msg.as_string()
    server.sendmail(sender, receiver, text)
    server.quit()

def main():
    # Read from stdin
    error_message = sys.stdin.read()
    if error_message:
        send_email(subject, error_message, sender_email, receiver_email)

if __name__ == "__main__":
    main()