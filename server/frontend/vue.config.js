const webpack = require('webpack');

module.exports = {
    configureWebpack: {
        resolve: {
            fallback: {
                buffer: require.resolve('buffer'),
            },
        },
        plugins: [
            new webpack.ProvidePlugin({
                Buffer: ['buffer', 'Buffer'],
            }),
        ],
    },
};
