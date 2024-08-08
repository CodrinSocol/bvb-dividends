import React, {ReactElement} from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import {BrowserRouter, Route, Routes} from "react-router-dom";
import {Layout} from "./components/Layout";
import {LandingPage} from "./pages/LandingPage";
import {AccountPage} from "./pages/Account";
import {Page} from "./pages/Page";
import {NotFound} from "./pages/NotFound";

export default function App(): ReactElement {
    return (
        <BrowserRouter>
            <Layout>
                <Routes>
                    <Route path="/" element={<LandingPage />} />
                    <Route path="/account" element={<AccountPage />} />
                    <Route path="/asd" element={<Page />} />
                    <Route path="*" element={<NotFound />} />
                </Routes>
            </Layout>
        </BrowserRouter>
    )
}
const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);
root.render(<App />);

