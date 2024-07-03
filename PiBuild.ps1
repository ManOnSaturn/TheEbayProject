$env:GOOS = "linux"
$env:GOARCH = "arm"
$env:GOARM = "7"
go build -o Scraper
wsl sshpass -p mattia scp -o StrictHostKeyChecking=no ./Scraper pi@37.182.98.146:/home/pi