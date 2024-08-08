import {ReactElement} from "react";
import {Header} from "./Header";

/** Wrapper component that adds the {@link Header} component to the top of every page.
 *
 * @params children: {@link ReactElement} - the children components to render
 *
 * @returns ReactElement: the Layout component
 * */
export function Layout({ children }: { children: ReactElement }): ReactElement {
    return (
        <div className={"w-[100vw] h-[100vh]"}>
            <Header/>
            <div className={"w-full h-[calc(100%-64px)] flex"}>
                {children}
            </div>
        </div>
    )
}