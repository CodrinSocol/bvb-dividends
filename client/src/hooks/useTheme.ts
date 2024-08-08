import {useState} from "react";

export function useTheme(): { theme: string; toggleTheme: () => void } {
    const [theme, setTheme] = useState("light");

    function toggleTheme() {
        const root = document.getElementById("app-root");
        if (root) {
            root.getAttribute('data-theme') === "light" ? root.setAttribute('data-theme', "dark") : root.setAttribute('data-theme', "light");
        }
        setTheme(theme === "light" ? "dark" : "light");
    }

    return { theme, toggleTheme };
}