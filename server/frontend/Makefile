rebuild:
	rm -rf node_modules
	rm -rf dist
	npm cache clean --force
	npm install --omit=dev
	npm prune
	npm run build