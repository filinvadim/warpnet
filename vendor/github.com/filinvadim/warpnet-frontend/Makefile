rebuild:
	npm config set timeout 60000
	rm -rf node_modules
	rm -rf dist
	npm cache clean --force
	npm install --omit=dev --prefer-offline --no-audit
	npm prune
	npm run build

setup-hooks:
	git config core.hooksPath .githooks
