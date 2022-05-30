import React from 'react';

interface Props {
    children: React.ReactNode;
    [x: string]: any
}


interface FormFieldProps {
    children: React.ReactNode;
    [x: string]: any
}
function FormField (props: FormFieldProps) {
    const { children, ...rest } = props;
    return (
        <div {...rest} className="flex flex-col mb-4">
            {children}
        </div>
    );
}


function Form(props: Props) {
    const { children, ...rest } = props;
    return (
        <form {...rest}>
            {children}
        </form>
    );
}

Form.Field = FormField;

export default Form;