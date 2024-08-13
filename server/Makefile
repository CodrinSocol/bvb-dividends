start-local-dependencies:
	docker-compose --project-directory=local-support up -d --renew-anon-volumes

stop-local-dependencies:
	docker-compose --project-directory=local-support down

refresh-secrets:
	./env/refresh_secrets.sh
