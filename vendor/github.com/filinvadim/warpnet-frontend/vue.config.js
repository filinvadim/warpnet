const webpack = require('webpack');

module.exports = {
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
