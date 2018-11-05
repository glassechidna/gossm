import * as React from "react";
import * as _ from "lodash";

export class Home extends React.Component<{}, {}> {
    render() {
        return <>
            <div className="container">
            </div>
        </>;
    }
}

function websocketUrl(): string {
    let loc = window.location;
    let newUri: string;

    if (loc.protocol === "https:") {
        newUri = "wss:";
    } else {
        newUri = "ws:";
    }
    newUri += "//" + loc.host;

    return newUri;
}
