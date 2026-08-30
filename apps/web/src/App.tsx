import { Route, Routes } from 'react-router-dom';
import { Layout } from './components/Layout';
import { DividendCalendar } from './pages/DividendCalendar';
import { CompanyDividends } from './pages/CompanyDividends';
import { NotFound } from './pages/NotFound';

export function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<DividendCalendar />} />
        <Route path="/companies/:symbol" element={<CompanyDividends />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </Layout>
  );
}
