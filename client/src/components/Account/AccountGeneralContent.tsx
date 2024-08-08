import {ReactElement} from "react";

export function AccountGeneralContent(): ReactElement {
    return (
        <div className={"flex flex-col gap-6 w-hull h-full"}>
            <h2 className={"flex flex-row text-2xl font-bold w-full justify-center"}>Account Settings</h2>

            <div className={"flex flex-col gap-6"}>
                <div className={"flex flex-col "}>
                    <label htmlFor="first-name-input">Change your First Name</label>
                    <input id={'first-name-input'} type="text" placeholder="First Name" className={"input input-sm input-ghost max-w-[250px]"}/>
                </div>
                <div className={"flex flex-col "}>
                    <label htmlFor="last-name-input">Change your Last Name</label>
                    <input id={'last-name-input'} type="text" placeholder="Last Name" className={"input input-sm max-w-[250px]"}/>
                </div>
                {/*<input id={'last-name-input'} type="text" placeholder="Last Name" className={"input input-sm max-w-[250px]"}/>*/}
            </div>

            <div className={"flex flex-col "}>
                <label htmlFor="email-input">Change your Email Address</label>
                <input id={'email-input'} type="email" placeholder="Email Address" className={"input input-sm max-w-[250px]"}/>
            </div>
        </div>)
}