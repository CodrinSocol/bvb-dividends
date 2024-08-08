import {ReactElement} from "react";
import {MdAccountCircle} from "react-icons/md";
import { useNavigate } from "react-router-dom";
import logoPath from "../logo-placeholder.png";


export function Header(): ReactElement {
    const navigate = useNavigate();

    function redirectTo(path: string = "") {
        navigate(`/${path}`);
    }

    return (
        <div className="flex h-16 w-full flex-row items-center justify-between bg-red-200 px-4">
            <div className={"flex flex-row gap-4 items-center"}>
                <img onClick={_ => redirectTo()} src={logoPath} alt="logo" className="h-16 cursor-pointer"/>
                <div className={"flex flex-row gap-5"}>
                    <span className={"cursor-pointer"}>Dividende Active</span>
                    <span className={"cursor-pointer"}>Istoric Dividende</span>
                    <span className={"cursor-pointer"}>Newsletter</span>
                    <span className={"cursor-pointer"}>About Us</span>
                </div>
            </div>
            <div className={"flex flex-row gap-6"}>
                <input type={"search"} className={"cursor-text"} placeholder={"search company"}/>
                <MdAccountCircle size={36}
                                 className={"cursor-pointer"}
                                 onClick={_ => redirectTo("account")}/>
            </div>
        </div>
    )
}