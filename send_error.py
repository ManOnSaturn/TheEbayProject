import sys
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart

# Configuration
sender_email = "mattiarip@gmail.com"
receiver_email = "mattiarip@gmail.com"
subject = "Error from Golang scraper"
smtp_server = "smtp.gmail.com"
smtp_port = 587
smtp_username = "mattiarip@gmail.com"
smtp_password = "chpiwikktegwtdzd"

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