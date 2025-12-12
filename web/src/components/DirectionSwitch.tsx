import React from "react";

interface Props {
    direction: "A_to_B" | "B_to_A";
    onChange: (dir: "A_to_B" | "B_to_A") => void;
}

const DirectionSwitch: React.FC<Props> = ({ direction, onChange }) => {
    const isAToB = direction === "A_to_B";

    return (
        <div className="direction-switch">
            <button
                className={"dir-btn" + (isAToB ? " active" : "")}
                onClick={() => onChange("A_to_B")}
            >
                A → B
            </button>
            <button
                className={"dir-btn" + (!isAToB ? " active" : "")}
                onClick={() => onChange("B_to_A")}
            >
                A ← B
            </button>
        </div>
    );
};

export default DirectionSwitch;
