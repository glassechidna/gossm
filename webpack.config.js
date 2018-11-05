const Webpack = require("webpack");
const Glob = require("glob");
const path = require("path");
const CopyWebpackPlugin = require("copy-webpack-plugin");
const MiniCssExtractPlugin = require("mini-css-extract-plugin");
const ManifestPlugin = require("webpack-manifest-plugin");
const CleanObsoleteChunks = require('webpack-clean-obsolete-chunks');
const UglifyJsPlugin = require("uglifyjs-webpack-plugin");
const LiveReloadPlugin = require('webpack-livereload-plugin');

const configurator = {
    entries: function(){
        return {
            application: [
                './assets/css/application.scss',
                './assets/js/application.tsx',
            ],
        };
    },

    plugins() {
        var plugins = [
            new CleanObsoleteChunks(),
            new MiniCssExtractPlugin({filename: "[name].[contenthash].css"}),
            new CopyWebpackPlugin([{from: "./assets",to: ""}], {copyUnmodified: true,ignore: ["css/**", "js/**"] }),
            new Webpack.LoaderOptionsPlugin({minimize: true,debug: false}),
            new ManifestPlugin({fileName: "manifest.json"})
        ];

        return plugins
    },

    moduleOptions: function() {
        return {
            rules: [
                {
                    test: /\.s[ac]ss$/,
                    use: [
                        MiniCssExtractPlugin.loader,
                        { loader: "css-loader", options: {sourceMap: true}},
                        { loader: "sass-loader", options: {sourceMap: true}}
                    ]
                },
                { test: /\.tsx?$/,loader: "awesome-typescript-loader",exclude: /node_modules/ },
                { test: /\.(woff|woff2|ttf|svg)(\?v=\d+\.\d+\.\d+)?$/,use: "url-loader"},
                { test: /\.eot(\?v=\d+\.\d+\.\d+)?$/,use: "file-loader" },
                { test: /\.go$/, use: "gopherjs-loader"}
            ]
        }
    },

    buildConfig: function(){
        const env = process.env.NODE_ENV || "development";

        var config = {
            mode: env,
            entry: configurator.entries(),
            output: {filename: "[name].[hash].js", path: `${__dirname}/public/assets`},
            plugins: configurator.plugins(),
            module: configurator.moduleOptions(),
            devtool: 'cheap-module-eval-source-map',
            resolve: {
                // Add `.ts` and `.tsx` as a resolvable extension.
                extensions: [".ts", ".tsx", ".js", ".jsx"]
            },
        }

        if( env === "development" ){
            config.plugins.push(new LiveReloadPlugin({appendScriptTag: true}))
            return config
        }

        const uglifier = new UglifyJsPlugin({
            uglifyOptions: {
                beautify: false,
                mangle: {keep_fnames: true},
                output: {comments: false},
                compress: {}
            }
        })

        config.optimization = {
            minimizer: [uglifier]
        }

        return config
    }
}

module.exports = configurator.buildConfig()
