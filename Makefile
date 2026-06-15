.PHONY: deploy
deploy:
	@echo "Stopping old container..."
	-sudo docker stop portfolio-app
	-sudo docker rm portfolio-app
	@echo "Building new binary..."
	sudo docker build -t ssh-portfolio:latest .
	@echo "Launching updated portfolio..."
	sudo docker run -d \
	  --name portfolio-app \
	  --restart unless-stopped \
	  -v $$(pwd)/ssh_keys:/home/appuser/.ssh \
	  -p 1000:23234 \
	  ssh-portfolio:latest

