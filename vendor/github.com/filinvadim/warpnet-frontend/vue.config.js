const webpack = require('webpack');

module.exports = {
    devServer: {
        overlay: {
            warnings: true,
            errors: true
        }
    },
    configureWebpack: {
        resolve: {
            fallback: {
                buffer: require.resolve('buffer'),
                "fs": false,
            },
        },
        plugins: [
            new webpack.ProvidePlugin({
                Buffer: ['buffer', 'Buffer'],
            }),
        ],
    },
};
