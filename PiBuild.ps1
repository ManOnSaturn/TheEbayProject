$env:GOOS = "linux"
$env:GOARCH = "arm64"
go build -o Scraper
scp -o StrictHostKeyChecking=no ./Scraper mattia@192.168.188.45:/home/mattia/ebay
scp -o StrictHostKeyChecking=no ./start_scraper.sh mattia@192.168.188.45:/home/mattia/ebay
scp -o StrictHostKeyChecking=no ./send_error.py mattia@192.168.188.45:/home/mattia/ebay