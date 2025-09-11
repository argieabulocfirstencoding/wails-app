import { useState } from "react";
import logo from "./assets/images/logo-universal.png";
import "./App.scss";
import "./styles/style.scss";
import { Greet } from "../wailsjs/go/app/App";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import Home from "./pages/Main/Home";
import Login from "./pages/Auth/Login";

function App() {
  const [resultText, setResultText] = useState(
    "Please enter your name below 👇"
  );
  const [name, setName] = useState("");
  const updateName = (e: any) => setName(e.target.value);
  const updateResultText = (result: string) => setResultText(result);

  function greet() {
    Greet(name).then(updateResultText);
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<Home />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
