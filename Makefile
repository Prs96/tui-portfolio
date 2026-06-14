.PHONY: deploy

deploy:
	@echo "Stopping old container..."
	-docker stop portfolio-app
	-docker rm portfolio-app
	@echo "Building new binary..."
	docker build -t ssh-portfolio:latest .
	@echo "Launching updated portfolio..."
	docker run -d \
	  --name portfolio-app \
	  --restart unless-stopped \
	  -v $$(pwd)/ssh_keys:/home/appuser/.ssh \
	  -p 23234:23234 \
	  ssh-portfolio:latest
