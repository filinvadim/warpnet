rebuild:
	rm -rf node_modules
	rm -rf dist
	npm cache clean --force
	npm install --production
	npm prune
	npm run build