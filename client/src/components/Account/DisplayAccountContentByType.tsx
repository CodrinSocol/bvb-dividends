import {ReactElement} from "react";
import {AccountGeneralContent} from "./AccountGeneralContent";

export interface DisplayAccountContentByTypeProps {
    selectedTab: string;
}

export function DisplayAccountContentByType({selectedTab}: DisplayAccountContentByTypeProps): ReactElement {

    switch(selectedTab) {
        case "general": return <AccountGeneralContent />
        case "contact": return <div>Contact</div>
        default: return <div>Default</div>
    }
}