import React from "react";
import Dashboard from "./pages/Dashboard";
import { ThemeProvider } from "./context/ThemeContext";

const App: React.FC = () => {
    return (
        <ThemeProvider>
            <Dashboard />
        </ThemeProvider>
    );
};

export default App;
