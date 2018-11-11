import * as React from "react";
import * as _ from "lodash";

type InvocationStatus = "Pending"|"InProgress"|"Delayed"|"Success"|"Cancelled"|"TimedOut"|"Failed"|"Cancelling";

interface InvocationCardProps {
    Status: InvocationStatus;
    InstanceId: string;
    Stdout: string;
    Stderr: string;
}

class InvocationCard extends React.Component<InvocationCardProps, {}> {
    render() {
        return <div className={`card ${this.cardClass()} mb-3`} style={{maxWidth: "20rem"}}>
            <div className="card-header">{this.props.InstanceId}</div>
            <div className="card-body">
                <pre>
                    {this.props.Stdout}
                    {this.props.Stderr}
                </pre>
            </div>
        </div>;
    }

    cardClass(): string {
        switch (this.props.Status) {
            case "Pending": return "border-secondary";
            case "InProgress": return "border-secondary";
            case "Delayed": return "border-secondary";
            case "Success": return "border-success";
            case "Cancelled": return "border-danger";
            case "TimedOut": return "border-danger";
            case "Failed": return "border-danger";
            case "Cancelling": return "border-danger";
        }
        return "";
    }
}

interface InvocationResp {
    Invocations: { [key: string]: InvocationCardProps };
}

export class Home extends React.Component<{}, {Invocations: InvocationCardProps[] }> {
    constructor(props) {
        super(props);
        this.state = {Invocations: []};
    }

    async componentDidMount() {
        let commandId = "3528c58b-65a8-443d-850b-d0057d73ceb7";
        let resp = await fetch(`/api/invocations?commandId=${commandId}`);
        let json: InvocationResp = await resp.json();
        this.setState((s, p) => {
            return {...s, Invocations: _.values(json.Invocations)};
        })
    }

    render() {
        return <>
            <div className="container">
                <div className="row">
                    <div className="col-sm">
                        Hello, WOrld
                        {this.state.Invocations.map(icp => <InvocationCard key={icp.InstanceId} {...icp} />)}
                    </div>
                </div>
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
