import {ReactElement, useState} from "react";
import {DisplayAccountContentByType} from "../components/Account/DisplayAccountContentByType";

export function AccountPage(): ReactElement {
    const [selectedTab, setSelectedTab] = useState('general')

    return (
        <div className="h-full w-full flex flex-row gap-2">
            <div className={"h-full w-[100px] justify-start bg-gray-50 flex flex-col gap-2"}>
                <span className={"cursor-pointer"} onClick={_ => setSelectedTab('general')}>General</span>
                <span className={"cursor-pointer"} onClick={_ => setSelectedTab('contact')}>Contact Us</span>
                <span className={"cursor-pointer"}> Sterge Contul</span>
            </div>
            <div className={"h-full w-[calc(100%-120px)]"}>
                <DisplayAccountContentByType selectedTab={selectedTab} />
            </div>
        </div>
    )
}