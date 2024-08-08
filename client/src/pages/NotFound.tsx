import {ReactElement} from "react";

export function NotFound(): ReactElement {
    return (
        <div className={"w-full h-full flex justify-center items-center text-3xl text-green-400"}>
            <div className={'text-green-400'}>Not Found</div>
        </div>
    )
}